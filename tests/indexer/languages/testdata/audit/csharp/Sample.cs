namespace App.Core
{
    public enum Role
    {
        Admin,
        Guest
    }

    public interface IRepository
    {
        string Find(int id);
    }

    public struct Coordinate
    {
        public int X;
        public int Y;
    }

    public record UserDto(string Name);

    public class UserService : IRepository
    {
        private readonly string _prefix;
        public int Count { get; set; }

        public UserService(string prefix)
        {
            _prefix = prefix;
        }

        public string Find(int id)
        {
            return Format(id);
        }

        private string Format(int id)
        {
            var label = _prefix;
            return label;
        }
    }
}
